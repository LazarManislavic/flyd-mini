<%@ Page Title="" Language="C#" MasterPageFile="~/Site.Master" AutoEventWireup="true" CodeBehind="Narudzbine.aspx.cs" Inherits="WebApplication6.Account.Narudzbine" %>
<asp:Content ID="Content1" ContentPlaceHolderID="MainContent" runat="server">

    <asp:Label ID="Label1" runat="server" Text="Sifra proizvoda"></asp:Label><asp:DropDownList ID="DropDownList1" runat="server" DataTextField="Sifra" DataValueField="Sifra" DataSourceID="SqlDataSource1"></asp:DropDownList><asp:SqlDataSource runat="server" ID="SqlDataSource1" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra] FROM [proizvodi]"></asp:SqlDataSource><br />
    <asp:Label ID="Label2" runat="server" Text="Kolicina"></asp:Label><asp:TextBox ID="TextBox2" runat="server"></asp:TextBox><br />
    <asp:Label ID="Label3" runat="server" Text="Zeljena cena za proizvod:"></asp:Label><asp:TextBox ID="TextBox1" runat="server"></asp:TextBox><br />
    <asp:Button ID="Button1" runat="server" Text="Button" OnClick="Button1_Click" /><asp:Label ID="ErrorLabel1" runat="server" Text="Label"></asp:Label>

</asp:Content>
